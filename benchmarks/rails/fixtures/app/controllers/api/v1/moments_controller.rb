module Api
  module V1
    class MomentsController < ApplicationController
      def index
        moments = Moment.all
        render json: moments
      end

      def create
        moment = Moment.create(moment_params)
        render json: moment, status: :created
      end

      private

      def moment_params
        params.require(:moment).permit(:title, :body)
      end
    end
  end
end
