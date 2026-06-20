module Api
  module V1
    class TokensController < ApplicationController
      def signup
        token = TokenGenerator.generate(params[:email], params[:password])
        user = User.find_by(email: params[:email])
        render json: { token: token, user_id: user.id }
      end
    end
  end
end
